import { useEffect, useState } from 'react'
import { Card, Row, Col, Statistic, Button, Space, Tag, Tooltip, Badge, Table, Modal, Form, Input, message, Spin, Typography } from 'antd'
import { ReloadOutlined, SafetyCertificateOutlined, EditOutlined, EyeOutlined, FileTextOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import SwaggerUI from 'swagger-ui-react'
import 'swagger-ui-react/swagger-ui.css'
import api from '../api/kong'

const API_BASE = import.meta.env.VITE_API_BASE || '/api'

interface ServiceHealth {
  service_id: string
  service_name: string
  health_url: string | null
  doc_url: string | null
  status: 'healthy' | 'unhealthy' | 'unreachable' | 'unknown'
  latency_ms: number
  error_message: string | null
  last_check_at: string | null
}

const statusConfig = {
  healthy: { color: '#52c41a', label: '健康', icon: '✔' },
  unhealthy: { color: '#ff4d4f', label: '異常', icon: '✘' },
  unreachable: { color: '#ff4d4f', label: '無法連線', icon: '✘' },
  unknown: { color: '#8c8c8c', label: '未知', icon: '?' },
}

async function apiGet(path: string) {
  const token = localStorage.getItem('kgo_token')
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

async function apiPost(path: string, body?: any) {
  const token = localStorage.getItem('kgo_token')
  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

async function apiPut(path: string, body: any) {
  const token = localStorage.getItem('kgo_token')
  const res = await fetch(`${API_BASE}${path}`, {
    method: 'PUT',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

function StatusBadge({ status }: { status: string }) {
  const cfg = statusConfig[status as keyof typeof statusConfig] || statusConfig.unknown
  return (
    <Badge
      status={status === 'healthy' ? 'success' : status === 'unknown' ? 'default' : 'error'}
      text={<span style={{ color: cfg.color, fontWeight: 600 }}>{cfg.label}</span>}
    />
  )
}

export default function HealthPortal() {
  const [services, setServices] = useState<ServiceHealth[]>([])
  const [summary, setSummary] = useState<{ total: number, healthy: number, unhealthy: number, unreachable: number, unknown: number }>({ total: 0, healthy: 0, unhealthy: 0, unreachable: 0, unknown: 0 })
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState(false)
  const [editModal, setEditModal] = useState(false)
  const [editing, setEditing] = useState<ServiceHealth | null>(null)
  const [docModal, setDocModal] = useState(false)
  const [docUrl, setDocUrl] = useState<string>('')
  const [form] = Form.useForm()
  const [saving, setSaving] = useState(false)

  const fetchHealth = () => {
    setLoading(true)
    Promise.all([
      apiGet('/health/services').catch(() => []),
      apiGet('/health/summary').catch(() => ({ total: 0, healthy: 0, unhealthy: 0, unreachable: 0, unknown: 0 }))
    ]).then(([svcs, sums]) => {
      setServices(svcs as ServiceHealth[])
      setSummary(sums as typeof summary)
    }).finally(() => setLoading(false))
  }

  useEffect(() => { fetchHealth() }, [])

  const handleCheck = async () => {
    setChecking(true)
    try {
      await apiPost('/health/check', {})
      message.success('健康檢查完成')
      fetchHealth()
    } catch (e: any) {
      message.error('執行失敗: ' + (e.message || '未知錯誤'))
    } finally {
      setChecking(false)
    }
  }

  const openEdit = (svc: ServiceHealth) => {
    setEditing(svc)
    form.setFieldsValue({ health_url: svc.health_url, doc_url: svc.doc_url })
    setEditModal(true)
  }

  const openDocModal = (url: string) => {
    setDocUrl(url)
    setDocModal(true)
  }

  const handleEditSave = async () => {
    try {
      const values = await form.validateFields()
      setSaving(true)
      await apiPut(`/health/services/${editing!.service_id}`, {
        health_url: values.health_url || null,
        doc_url: values.doc_url || null
      })
      message.success('設定已儲存')
      setEditModal(false)
      fetchHealth()
    } catch (e: any) {
      if (e.errorFields) return
      message.error('儲存失敗: ' + (e.message || '未知錯誤'))
    } finally {
      setSaving(false)
    }
  }

  const columns: ColumnsType<ServiceHealth> = [
    {
      title: '狀態',
      key: 'status',
      width: 100,
      render: (_, r) => (
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <div style={{
            width: 12, height: 12, borderRadius: '50%',
            background: statusConfig[r.status as keyof typeof statusConfig]?.color || '#8c8c8c',
            boxShadow: r.status === 'healthy' ? `0 0 8px ${statusConfig.healthy.color}` : 'none'
          }} />
          <StatusBadge status={r.status} />
        </div>
      )
    },
    { title: '服務名稱', dataIndex: 'service_name', key: 'service_name', render: v => <Tag color="blue">{v}</Tag> },
    {
      title: '健康檢查 URL',
      dataIndex: 'health_url',
      key: 'health_url',
      ellipsis: true,
      render: v => v ? <code style={{ fontSize: 11 }}>{v}</code> : <span style={{ color: '#8c8c8c' }}>未設定</span>
    },
    {
      title: '延遲',
      key: 'latency',
      width: 100,
      render: (_, r) => r.latency_ms > 0 ? <span style={{ color: r.latency_ms > 3000 ? '#ff4d4f' : r.latency_ms > 1000 ? '#faad14' : '#52c41a' }}>{r.latency_ms}ms</span> : '-'
    },
    {
      title: '錯誤',
      dataIndex: 'error_message',
      key: 'error_message',
      ellipsis: true,
      render: v => v ? <Tooltip title={v}><span style={{ color: '#ff4d4f', fontSize: 12 }}>{v}</span></Tooltip> : '-'
    },
    {
      title: '文件',
      key: 'doc',
      width: 100,
      render: (_, r) => r.doc_url ? (
        <Space size={4}>
          <Button size="small" icon={<FileTextOutlined />} onClick={() => openDocModal(r.doc_url!)}>Swagger</Button>
          <a href={r.doc_url} target="_blank" rel="noopener noreferrer"><EyeOutlined /></a>
        </Space>
      ) : <span style={{ color: '#8c8c8c', fontSize: 12 }}>未設定</span>
    },
    {
      title: '最後檢查',
      key: 'last_check',
      width: 160,
      render: (_, r) => r.last_check_at ? new Date(r.last_check_at).toLocaleString() : '-'
    },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_, r) => (
        <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>設定</Button>
      )
    }
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 24 }}>
        <h1>健康狀態監控</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchHealth} loading={loading}>刷新</Button>
          <Button type="primary" icon={<SafetyCertificateOutlined />} onClick={handleCheck} loading={checking}>執行檢查</Button>
        </Space>
      </div>

      {/* Summary Cards */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card size="small">
            <Statistic title="總服務數" value={summary.total} valueStyle={{ color: '#1890ff' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic title="健康" value={summary.healthy} valueStyle={{ color: '#52c41a' }}
              prefix={<span style={{ color: '#52c41a', marginRight: 4 }}>●</span>} />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic title="異常" value={summary.unhealthy + summary.unreachable} valueStyle={{ color: '#ff4d4f' }}
              prefix={<span style={{ color: '#ff4d4f', marginRight: 4 }}>●</span>} />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic title="未知" value={summary.unknown} valueStyle={{ color: '#8c8c8c' }} />
          </Card>
        </Col>
      </Row>

      {/* Health Status Grid - Cards */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        {loading ? (
          <Col span={24} style={{ textAlign: 'center', padding: 40 }}>
            <Spin size="large" />
          </Col>
        ) : services.length === 0 ? (
          <Col span={24}>
            <Card style={{ textAlign: 'center', color: '#8c8c8c' }}>
              尚無健康檢查設定，請在「服務」頁面設定健康檢查 URL
            </Card>
          </Col>
        ) : services.map(svc => {
          const cfg = statusConfig[svc.status as keyof typeof statusConfig] || statusConfig.unknown
          return (
            <Col key={svc.service_id} span={6}>
              <Card
                size="small"
                style={{
                  borderLeft: `4px solid ${cfg.color}`,
                  boxShadow: svc.status === 'healthy' ? `0 0 12px ${cfg.color}40` : 'none'
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                  <Tag color="blue">{svc.service_name}</Tag>
                  <div style={{
                    width: 14, height: 14, borderRadius: '50%',
                    background: cfg.color,
                    boxShadow: svc.status === 'healthy' ? `0 0 10px ${cfg.color}` : 'none',
                    animation: svc.status === 'healthy' ? 'pulse 2s infinite' : 'none'
                  }} />
                </div>
                <div style={{ fontSize: 12, color: cfg.color, fontWeight: 600, marginBottom: 4 }}>{cfg.label}</div>
                {svc.latency_ms > 0 && <div style={{ fontSize: 11, color: '#8c8c8c' }}>延遲: {svc.latency_ms}ms</div>}
                {svc.error_message && <div style={{ fontSize: 11, color: '#ff4d4f', marginTop: 4 }}>{svc.error_message}</div>}
                <div style={{ marginTop: 8, display: 'flex', gap: 8 }}>
                  {svc.doc_url && (
                    <a onClick={() => openDocModal(svc.doc_url!)} style={{ fontSize: 11, cursor: 'pointer' }}>📄 Swagger</a>
                  )}
                  <a onClick={() => openEdit(svc)} style={{ fontSize: 11, cursor: 'pointer' }}>⚙️ 設定</a>
                </div>
                <style>{`@keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.5} }`}</style>
              </Card>
            </Col>
          )
        })}
      </Row>

      {/* Table of all services */}
      <Card title="詳細列表" size="small">
        <Table
          columns={columns}
          dataSource={services as any[]}
          rowKey="service_id"
          loading={loading}
          pagination={{ pageSize: 10, size: 'small' }}
          size="small"
        />
      </Card>

      {/* Edit Modal */}
      <Modal
        title={`健康檢查設定 — ${editing?.service_name}`}
        open={editModal}
        onOk={handleEditSave}
        confirmLoading={saving}
        onCancel={() => setEditModal(false)}
        okText="儲存"
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="health_url" label="健康檢查 URL" tooltip="Service 的 /health 或 /status endpoint">
            <Input placeholder="https://api.example.com/health" />
          </Form.Item>
          <Form.Item name="doc_url" label="API 文件 URL" tooltip="OpenAPI/Swagger 文件連結">
            <Input placeholder="https://api.example.com/docs" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Swagger UI Modal */}
      <Modal
        title={<><FileTextOutlined style={{ marginRight: 8 }} />API 文件 — {editing?.service_name}</>}
        open={docModal}
        onCancel={() => setDocModal(false)}
        footer={null}
        width="80%"
        style={{ top: 20 }}
      >
        <div style={{ height: '70vh', overflow: 'auto' }}>
          {docUrl ? (
            <SwaggerUI url={docUrl} />
          ) : (
            <div style={{ textAlign: 'center', color: '#8c8c8c', padding: 40 }}>尚無文件 URL</div>
          )}
        </div>
      </Modal>
    </div>
  )
}

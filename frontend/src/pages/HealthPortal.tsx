import { useEffect, useState } from 'react'
import { Card, Row, Col, Statistic, Button, Space, Tag, Tooltip, Badge, Table, Modal, Form, Input, message, Spin, Typography, Select, Alert } from 'antd'
import { ReloadOutlined, SafetyCertificateOutlined, SyncOutlined, CheckCircleOutlined, CloseCircleOutlined, MinusCircleOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '../api/kong'
import type { KongUpstream, UpstreamHealth, TargetHealth } from '../api/kong'

const API_BASE = import.meta.env.VITE_API_BASE || '/api'

interface UpstreamHealthData extends KongUpstream {
  targets: TargetHealth[]
  healthyCount: number
  unhealthyCount: number
  overallStatus: 'healthy' | 'unhealthy' | 'partial' | 'unknown'
}

const statusConfig = {
  healthy: { color: '#52c41a', label: '健康', icon: <CheckCircleOutlined style={{ color: '#52c41a' }} /> },
  unhealthy: { color: '#ff4d4f', label: '異常', icon: <CloseCircleOutlined style={{ color: '#ff4d4f' }} /> },
  partial: { color: '#faad14', label: '部分異常', icon: <MinusCircleOutlined style={{ color: '#faad14' }} /> },
  unknown: { color: '#8c8c8c', label: '未知', icon: <MinusCircleOutlined style={{ color: '#8c8c8c' }} /> },
}

async function apiGet(path: string) {
  const token = localStorage.getItem('cont_token')
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

function StatusBadge({ status }: { status: keyof typeof statusConfig }) {
  const cfg = statusConfig[status] || statusConfig.unknown
  return (
    <Space size={4}>
      {cfg.icon}
      <span style={{ color: cfg.color, fontWeight: 600 }}>{cfg.label}</span>
    </Space>
  )
}

function HealthDot({ healthy }: { healthy: boolean }) {
  return (
    <div style={{
      width: 10, height: 10, borderRadius: '50%',
      background: healthy ? '#52c41a' : '#ff4d4f',
      boxShadow: healthy ? '0 0 6px #52c41a' : 'none'
    }} />
  )
}

export default function HealthPortal() {
  const [upstreams, setUpstreams] = useState<UpstreamHealthData[]>([])
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState(false)
  const [detailModal, setDetailModal] = useState(false)
  const [selectedUpstream, setSelectedUpstream] = useState<UpstreamHealthData | null>(null)
  const [healthData, setHealthData] = useState<UpstreamHealth | null>(null)
  const [healthLoading, setHealthLoading] = useState(false)
  const [form] = Form.useForm()

  const fetchUpstreams = () => {
    setLoading(true)
    apiGet('/upstreams')
      .then(async data => {
        const list = Array.isArray(data) ? data : (data?.data || [])
        // fetch health data for each upstream concurrently
        const upstreamsWithHealth: UpstreamHealthData[] = await Promise.all(
          list.map(async (u: KongUpstream) => {
            try {
              const health: UpstreamHealth = await apiGet(`/upstreams/${u.id}/health`)
              const targets: TargetHealth[] = health?.targets || []
              const healthyCount = targets.filter((t: TargetHealth) => t.healthy).length
              const unhealthyCount = targets.filter((t: TargetHealth) => !t.healthy).length
              const overallStatus: UpstreamHealthData['overallStatus'] =
                healthyCount === 0 && unhealthyCount === 0
                  ? 'unknown'
                  : unhealthyCount === 0
                  ? 'healthy'
                  : healthyCount === 0
                  ? 'unhealthy'
                  : 'partial'
              return { ...u, targets, healthyCount, unhealthyCount, overallStatus }
            } catch {
              return {
                ...u,
                targets: [],
                healthyCount: 0,
                unhealthyCount: 0,
                overallStatus: 'unknown' as const,
              }
            }
          })
        )
        setUpstreams(upstreamsWithHealth)
      })
      .catch(() => {
        message.error('無法取得上游列表')
        setUpstreams([])
      })
      .finally(() => setLoading(false))
  }

  const fetchUpstreamHealth = (upstreamId: string) => {
    setHealthLoading(true)
    apiGet(`/upstreams/${upstreamId}/health`)
      .then((data: UpstreamHealth) => {
        setHealthData(data)
      })
      .catch(() => {
        message.error('無法取得健康狀態')
        setHealthData(null)
      })
      .finally(() => setHealthLoading(false))
  }

  useEffect(() => {
    fetchUpstreams()
  }, [])

  const openDetail = async (upstream: UpstreamHealthData) => {
    setSelectedUpstream(upstream)
    setDetailModal(true)
    fetchUpstreamHealth(upstream.id!)
  }

  const handleRefresh = () => {
    fetchUpstreams()
  }

  const handleCheckAll = async () => {
    setChecking(true)
    try {
      await Promise.all(upstreams.map(u => apiGet(`/upstreams/${u.id}/health`)))
      message.success('已刷新所有上游健康狀態')
      fetchUpstreams()
    } catch {
      message.error('部分上游健康檢查失敗')
    } finally {
      setChecking(false)
    }
  }

  const targetColumns: ColumnsType<TargetHealth> = [
    {
      title: '狀態',
      key: 'healthy',
      width: 80,
      render: (_, t) => <HealthDot healthy={t.healthy} />
    },
    { title: 'Target', dataIndex: 'target', key: 'target', render: v => <code style={{ fontSize: 12 }}>{v}</code> },
    { title: 'Host', dataIndex: 'host', key: 'host', render: v => <span style={{ color: '#555' }}>{v}</span> },
    { title: 'Port', dataIndex: 'port', key: 'port', width: 70 },
    { title: 'Weight', dataIndex: 'weight', key: 'weight', width: 70 },
    {
      title: 'Enabled',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: v => v ? <Tag color="green">是</Tag> : <Tag color="red">否</Tag>
    },
    {
      title: '健康狀態',
      key: 'status',
      width: 100,
      render: (_, t) => t.healthy
        ? <Tag color="success">健康</Tag>
        : <Tag color="error">異常</Tag>
    },
  ]

  const upstreamColumns: ColumnsType<UpstreamHealthData> = [
    {
      title: '狀態',
      key: 'status',
      width: 120,
      render: (_, u) => {
        const healthy = u.healthyCount
        const unhealthy = u.unhealthyCount
        const total = healthy + unhealthy
        if (total === 0) return <StatusBadge status="unknown" />
        if (unhealthy === 0) return <StatusBadge status="healthy" />
        if (healthy === 0) return <StatusBadge status="unhealthy" />
        return <StatusBadge status="partial" />
      }
    },
    { title: '名稱', dataIndex: 'name', key: 'name', render: v => <Tag color="blue">{v}</Tag> },
    { title: '演算法', dataIndex: 'algorithm', key: 'algorithm', width: 120 },
    {
      title: 'Targets',
      key: 'targets',
      width: 120,
      render: (_, u) => {
        const healthy = u.healthyCount
        const unhealthy = u.unhealthyCount
        const total = healthy + unhealthy
        if (total === 0) return <span style={{ color: '#8c8c8c' }}>無資料</span>
        return (
          <Space size={4}>
            <span style={{ color: '#52c41a' }}>● {healthy}</span>
            {unhealthy > 0 && <span style={{ color: '#ff4d4f' }}>● {unhealthy}</span>}
            <span style={{ color: '#8c8c8c' }}>/ {total}</span>
          </Space>
        )
      }
    },
    {
      title: 'Enabled',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: v => v ? <Tag color="green">是</Tag> : <Tag color="red">否</Tag>
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_, u) => (
        <Button size="small" type="link" onClick={() => openDetail(u)}>
          詳細
        </Button>
      )
    },
  ]

  // Summary stats
  const totalUpstreams = upstreams.length
  const healthyUpstreams = upstreams.filter(u => u.overallStatus === 'healthy').length
  const unhealthyUpstreams = upstreams.filter(u => u.overallStatus === 'unhealthy').length
  const partialUpstreams = upstreams.filter(u => u.overallStatus === 'partial').length
  const totalTargets = upstreams.reduce((sum, u) => sum + u.healthyCount + u.unhealthyCount, 0)
  const totalHealthyTargets = upstreams.reduce((sum, u) => sum + u.healthyCount, 0)

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 24 }}>
        <h1>API 後端狀態監控</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={handleRefresh} loading={loading}>刷新列表</Button>
          <Button type="primary" icon={<SafetyCertificateOutlined />} onClick={handleCheckAll} loading={checking}>
            執行健康檢查
          </Button>
        </Space>
      </div>

      {/* Summary Cards */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={4}>
          <Card size="small">
            <Statistic title="API 後端總數" value={totalUpstreams} valueStyle={{ color: '#1890ff' }} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title="健康 API 後端" value={healthyUpstreams} valueStyle={{ color: '#52c41a' }}
              prefix={<span style={{ color: '#52c41a', marginRight: 4 }}>●</span>} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title="異常 API 後端" value={unhealthyUpstreams} valueStyle={{ color: '#ff4d4f' }}
              prefix={<span style={{ color: '#ff4d4f', marginRight: 4 }}>●</span>} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title="部分異常" value={partialUpstreams} valueStyle={{ color: '#faad14' }}
              prefix={<span style={{ color: '#faad14', marginRight: 4 }}>●</span>} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title="所有 Target 總數" value={totalTargets} valueStyle={{ color: '#8c8c8c' }} />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic title="健康 Target" value={totalHealthyTargets} valueStyle={{ color: '#52c41a' }}
              prefix={<span style={{ color: '#52c41a', marginRight: 4 }}>●</span>} suffix={`/ ${totalTargets}`} />
          </Card>
        </Col>
      </Row>

      {loading ? (
        <Card style={{ textAlign: 'center', padding: 40 }}>
          <Spin size="large" />
          <div style={{ marginTop: 16, color: '#8c8c8c' }}>載入中...</div>
        </Card>
      ) : upstreams.length === 0 ? (
        <Card style={{ textAlign: 'center', color: '#8c8c8c', padding: 40 }}>
          尚無上游設定，請先在「上游」頁面建立 Upstream
        </Card>
      ) : (
        <>
          {/* Upstream Cards */}
          <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
            {upstreams.map(u => {
              const cfg = statusConfig[u.overallStatus] || statusConfig.unknown
              return (
                <Col key={u.id} span={6}>
                  <Card
                    size="small"
                    style={{
                      borderLeft: `4px solid ${cfg.color}`,
                      boxShadow: u.overallStatus === 'healthy' ? `0 0 12px ${cfg.color}40` : 'none'
                    }}
                    title={<Space><HealthDot healthy={u.overallStatus === 'healthy'} /><span>{u.name}</span></Space>}
                    extra={<Tag color={u.enabled ? 'green' : 'red'}>{u.enabled ? '啟用' : '停用'}</Tag>}
                  >
                    <div style={{ fontSize: 13, color: '#555', marginBottom: 4 }}>
                      演算法: <code>{u.algorithm || 'roundrobin'}</code>
                    </div>
                    <div style={{ fontSize: 12, marginBottom: 8 }}>
                      <Space size={4}>
                        <span style={{ color: '#52c41a' }}>● {u.healthyCount} 健康</span>
                        {u.unhealthyCount > 0 && <span style={{ color: '#ff4d4f' }}>● {u.unhealthyCount} 異常</span>}
                      </Space>
                    </div>
                    <Button size="small" type="link" onClick={() => openDetail(u)}>
                      查看詳情 →
                    </Button>
                  </Card>
                </Col>
              )
            })}
          </Row>

          {/* Upstreams Table */}
          <Card title="API 後端列表">
            <Table
              columns={upstreamColumns}
              dataSource={upstreams as any[]}
              rowKey="id"
              loading={loading}
              pagination={{ pageSize: 10, size: 'small' }}
              size="small"
            />
          </Card>
        </>
      )}

      {/* Upstream Detail Modal */}
      <Modal
        title={<Space><SafetyCertificateOutlined />API 後端健康狀態 — {selectedUpstream?.name}</Space>}
        open={detailModal}
        onCancel={() => { setDetailModal(false); setHealthData(null) }}
        footer={[
          <Button key="refresh" icon={<SyncOutlined spin={healthLoading} />} onClick={() => selectedUpstream && fetchUpstreamHealth(selectedUpstream.id!)}>
            刷新
          </Button>,
          <Button key="close" type="primary" onClick={() => setDetailModal(false)}>關閉</Button>
        ]}
        width={800}
      >
        {healthLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin size="large" />
            <div style={{ marginTop: 16 }}>正在獲取健康狀態...</div>
          </div>
        ) : healthData ? (
          <div>
            <Alert
              type={healthData.targets.every(t => t.healthy) ? 'success' : healthData.targets.some(t => t.healthy) ? 'warning' : 'error'}
              message={
                <Space>
                  {healthData.upstream_name}
                  — {healthData.targets.filter(t => t.healthy).length} / {healthData.targets.length} targets 健康
                </Space>
              }
              style={{ marginBottom: 16 }}
            />

            <Row gutter={16} style={{ marginBottom: 16 }}>
              <Col span={6}>
                <Statistic title="上游名稱" value={healthData.upstream_name} />
              </Col>
              <Col span={6}>
                <Statistic title="負載平衡" value={healthData.algorithm} />
              </Col>
              <Col span={6}>
                <Statistic title="Target 數" value={healthData.targets.length} />
              </Col>
              <Col span={6}>
                <Statistic
                  title="狀態"
                  value={healthData.targets.filter(t => t.healthy).length === healthData.targets.length ? '全部健康' : '有異常'}
                  valueStyle={{ color: healthData.targets.every(t => t.healthy) ? '#52c41a' : '#ff4d4f' }}
                />
              </Col>
            </Row>

            <Table
              columns={targetColumns}
              dataSource={healthData.targets}
              rowKey="id"
              pagination={false}
              size="small"
            />
          </div>
        ) : (
          <div style={{ textAlign: 'center', color: '#8c8c8c', padding: 40 }}>
            無法載入健康狀態資料
          </div>
        )}
      </Modal>
    </div>
  )
}
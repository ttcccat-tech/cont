import { useEffect, useState } from 'react'
import { Row, Col, Card, Statistic, Spin, Tag, message } from 'antd'
import { CheckCircleOutlined, ExclamationCircleOutlined, DownCircleOutlined } from '@ant-design/icons'
import { getStatus, getInfo } from '../api/kong'

interface KongStatus {
  database: { reachable: boolean }
  memory: {
    lua_shared_dicts?: Record<string, { allocated_slabs: string; capacity: string }>
    workers_lua_vms?: Array<{ pid: number; http_allocated_gc: string }>
  }
  server?: {
    connections_active: number
    total_requests: number
    connections_accepted: number
  }
  version?: string
}

interface KongRoot {
  version: string
  tagline: string
  plugins?: { enabled_in_cluster: string[] }
}

export default function Dashboard() {
  const [status, setStatus] = useState<KongStatus | null>(null)
  const [root, setRoot] = useState<KongRoot | null>(null)
  const [loading, setLoading] = useState(true)
  const [kongReachable, setKongReachable] = useState(false)

  useEffect(() => {
    const timer = setTimeout(() => {
      if (loading) setLoading(false)
    }, 5000)

    Promise.all([getStatus().catch(() => null), getInfo().catch(() => null)])
      .then(([s, i]) => {
        if (!s && !i) { setKongReachable(false); return }
        if (s) setStatus(s as KongStatus)
        if (i) setRoot(i as KongRoot)
        setKongReachable(true)
      })
      .catch(() => setKongReachable(false))
      .finally(() => { setLoading(false); clearTimeout(timer) })
    return () => clearTimeout(timer)
  }, [])

  if (loading) return <Spin size="large" style={{ display: 'flex', justifyContent: 'center', marginTop: 80 }} />

  if (!kongReachable || !status) {
    return (
      <div style={{ textAlign: 'center', marginTop: 80 }}>
        <ExclamationCircleOutlined style={{ fontSize: 48, color: '#faad14' }} />
        <h2 style={{ color: 'var(--text)' }}>無法連接 Cont Admin API</h2>
        <p style={{ color: 'var(--muted)' }}>請確認 Cont API 容器已在運行，端口 18081 可正常訪問</p>
      </div>
    )
  }

  const dbOk = status?.database?.reachable
  const version = root?.version || status?.version || '-'

  const activeConns = status?.server?.connections_active ?? '-'
  const totalReqs = status?.server?.total_requests ?? '-'

  const workerCount = status?.memory?.workers_lua_vms?.length ?? '-'
  const workerVMs = status?.memory?.workers_lua_vms ?? []

  const enabledPlugins = root?.plugins?.enabled_in_cluster ?? []

  return (
    <div>
      <h1 style={{ marginBottom: 24 }}>系統概覽</h1>
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={<span style={{ color: 'var(--muted)' }}>Cont 狀態</span>}
              valueRender={() => (
                <div>
                  <Tag icon={<CheckCircleOutlined />} color={dbOk ? 'green' : 'red'}>
                    {dbOk ? 'Database OK' : 'Database Error'}
                  </Tag>
                  <Tag icon={<CheckCircleOutlined />} color="blue" style={{ marginLeft: 8 }}>Workers OK</Tag>
                </div>
              )}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card><Statistic title={<span style={{ color: 'var(--muted)' }}>Cont 版本</span>} value={version} /></Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card><Statistic title={<span style={{ color: 'var(--muted)' }}>Workers</span>} value={workerCount} /></Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title={<span style={{ color: 'var(--muted)' }}>活躍連接</span>}
              value={activeConns}
              suffix={<span style={{ fontSize: 12, color: 'var(--muted)' }}>/ 總請求 {totalReqs}</span>}
            />
          </Card>
        </Col>
      </Row>

      {workerVMs.length > 0 && (
        <Card title="Worker 記憶體" style={{ marginTop: 16 }}>
          <Row gutter={[12, 12]}>
            {workerVMs.map((w, i) => (
              <Col xs={24} sm={12} lg={6} key={i}>
                <Tag color="cyan">PID {w.pid}</Tag>
                <span style={{ color: 'var(--text)', fontSize: 13 }}>GC: {w.http_allocated_gc}</span>
              </Col>
            ))}
          </Row>
        </Card>
      )}

      {enabledPlugins.length > 0 && (
        <Card title="已啟用插件" style={{ marginTop: 16 }}>
          {enabledPlugins.map(p => (
            <Tag key={p} color="purple" style={{ margin: 4 }}>{p}</Tag>
          ))}
        </Card>
      )}
    </div>
  )
}

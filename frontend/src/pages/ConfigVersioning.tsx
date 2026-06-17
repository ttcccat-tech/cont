import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, Card, Spin, Modal, message, Popconfirm, Statistic, Row, Col } from 'antd'
import {
  ReloadOutlined, CameraOutlined, RollbackOutlined, DiffOutlined,
  DeleteOutlined, EyeOutlined, WarningOutlined
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { getConfigSnapshots, createConfigSnapshot, rollbackSnapshot, diffSnapshots, deleteConfigSnapshot } from '../api/kong'

interface Snapshot {
  id: number
  version_label: string
  diff_from_prev: any
  actor_username: string
  actor_user_id: string
  created_at: string
}

interface DiffView {
  id1: number
  id2: number
  diff: {
    services: Array<{ op: string; name: string; item?: any; changes?: Record<string, { from: any; to: any }> }>
    routes: Array<{ op: string; name: string; item?: any; changes?: Record<string, { from: any; to: any }> }>
    plugins: Array<{ op: string; name: string; item?: any; changes?: Record<string, { from: any; to: any }> }>
    consumers: Array<{ op: string; name: string; item?: any; changes?: Record<string, { from: any; to: any }> }>
  }
}

const opColor = (op: string) => {
  if (op === 'add') return 'green'
  if (op === 'delete') return 'red'
  return 'blue'
}
const opLabel = (op: string) => {
  if (op === 'add') return '新增'
  if (op === 'delete') return '刪除'
  return '變更'
}

export default function ConfigVersioning() {
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [rollingBack, setRollingBack] = useState<number | null>(null)
  const [diffModal, setDiffModal] = useState<{ open: boolean; diff: DiffView['diff'] | null; title: string }>({
    open: false,
    diff: null,
    title: '',
  })
  const [selectedKeys, setSelectedKeys] = useState<string[]>([])

  const load = async () => {
    setLoading(true)
    try {
      const rows = await getConfigSnapshots(100)
      setSnapshots(Array.isArray(rows) ? rows : (rows?.data || []))
    } catch {
      message.error('載入快照失敗')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const handleCreateSnapshot = async () => {
    setCreating(true)
    try {
      const label = prompt('快照版本標籤（可輸入版本名稱或時間點描述）：', `v${new Date().toISOString().slice(0, 16).replace('T', ' ')}`)
      if (!label) { setCreating(false); return }
      await createConfigSnapshot(label)
      message.success('快照已建立')
      load()
    } catch {
      message.error('建立快照失敗')
    } finally {
      setCreating(false)
    }
  }

  const handleRollback = async (id: number) => {
    setRollingBack(id)
    try {
      const result = await rollbackSnapshot(id)
      if (result.success) {
        message.success(`已回滾至快照 #${id}`)
      }
      if (result.errors?.length) {
        message.warning(`回滾完成，但有 ${result.errors.length} 項錯誤`)
      }
    } catch {
      message.error('回滾請求失敗')
    } finally {
      setRollingBack(null)
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await deleteConfigSnapshot(id)
      message.success(`快照 #${id} 已刪除`)
      load()
    } catch {
      message.error('刪除失敗')
    }
  }

  const openDiff = async (id1: number, id2: number, label1: string, label2: string) => {
    try {
      const result = await diffSnapshots(id1, id2)
      setDiffModal({ open: true, diff: result.diff, title: `${label1} → ${label2}` })
    } catch {
      message.error('載入差異失敗')
    }
  }

  const openSelfDiff = async (snap: Snapshot) => {
    if (!snap.diff_from_prev) {
      message.info('此快照沒有差異記錄')
      return
    }
    setDiffModal({ open: true, diff: snap.diff_from_prev, title: `快照 #${snap.id} 差異` })
  }

  const renderDiffSection = (title: string, items: DiffView['diff']['services']) => {
    if (!items || items.length === 0) return null
    return (
      <div style={{ marginBottom: 16 }}>
        <b style={{ fontSize: 13, color: 'var(--highlight)' }}>{title}</b>
        {items.map((item, i) => (
          <Card
            key={i}
            size="small"
            style={{
              background: 'var(--primary)',
              border: `1px solid ${item.op === 'add' ? 'var(--green)' : item.op === 'delete' ? 'var(--red)' : 'var(--blue)'}`,
              marginTop: 4,
            }}
          >
            <Space>
              <Tag color={opColor(item.op)}>{opLabel(item.op)}</Tag>
              <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{item.name}</span>
            </Space>
            {item.changes && (
              <div style={{ marginTop: 4, fontSize: 11 }}>
                {Object.entries(item.changes).map(([k, v]) => (
                  <div key={k} style={{ color: 'var(--muted)' }}>
                    <span style={{ color: 'var(--text)' }}>{k}</span>
                    {' : '}
                    <span style={{ color: '#ff6b6b' }}>{JSON.stringify(v.from)}</span>
                    {' → '}
                    <span style={{ color: '#69db7c' }}>{JSON.stringify(v.to)}</span>
                  </div>
                ))}
              </div>
            )}
          </Card>
        ))}
      </div>
    )
  }

  const columns: ColumnsType<Snapshot> = [
    {
      title: '版本',
      dataIndex: 'version_label',
      key: 'version_label',
      render: v => <b style={{ color: 'var(--highlight)' }}>{v || '(未命名)'}</b>,
    },
    {
      title: '創建時間',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: v => (
        <span style={{ fontFamily: 'monospace', fontSize: 12, color: 'var(--muted)' }}>
          {new Date(v).toLocaleString('zh-TW', { hour12: false })}
        </span>
      ),
      sorter: (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    },
    {
      title: '操作者',
      dataIndex: 'actor_username',
      key: 'actor_username',
      width: 120,
      render: v => <Tag>{v || 'system'}</Tag>,
    },
    {
      title: '變更摘要',
      dataIndex: 'diff_from_prev',
      key: 'diff_from_prev',
      render: (_, r) => {
        if (!r.diff_from_prev) return <Tag>—</Tag>
        const d = r.diff_from_prev
        const total = (d.services?.length || 0) + (d.routes?.length || 0) + (d.plugins?.length || 0) + (d.consumers?.length || 0)
        return total > 0 ? (
          <Button size="small" icon={<EyeOutlined />} onClick={() => openSelfDiff(r)}>
            檢視 {total} 項變更
          </Button>
        ) : <Tag color="green">無變更</Tag>
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 260,
      render: (_, record) => (
        <Space>
          <Button
            size="small"
            icon={<DiffOutlined />}
            onClick={() => {
              const prev = snapshots.find(s => s.id === record.id - 1)
              if (!prev) { message.info('沒有更早的快照可對比'); return }
              const prevLabel = prev?.version_label || `#${prev.id}`
              openDiff(prev.id, record.id, prevLabel, record.version_label)
            }}
          >
            對比
          </Button>
          <Popconfirm
            title="確定要回滾至此快照？"
            description={<span style={{ color: 'var(--warning)' }}>当前 Cont 配置将被此快照覆盖</span>}
            onConfirm={() => handleRollback(record.id)}
            okText="確定回滾"
            cancelText="取消"
            okButtonProps={{ danger: true }}
          >
            <Button size="small" danger icon={<RollbackOutlined />} loading={rollingBack === record.id}>
              回滾
            </Button>
          </Popconfirm>
          <Popconfirm
            title="確定要刪除此快照？"
            description={<span style={{ color: 'var(--warning)' }}>此操作不可恢復</span>}
            onConfirm={() => handleDelete(record.id)}
            okText="刪除"
            cancelText="取消"
            okButtonProps={{ danger: true }}
          >
            <Button size="small" danger icon={<DeleteOutlined />}>
              刪除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16, alignItems: 'center' }}>
        <h1>Config Versioning</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>刷新</Button>
          <Button type="primary" icon={<CameraOutlined />} onClick={handleCreateSnapshot} loading={creating}>
            建立快照
          </Button>
        </Space>
      </div>

      {/* Info card */}
      <Card style={{ background: 'var(--secondary)', border: 'none', marginBottom: 16, padding: '12px 16px' }}>
        <Row gutter={24}>
          <Col>
            <Statistic
              title={<span style={{ color: 'var(--muted)', fontSize: 12 }}>總快照數</span>}
              value={snapshots.length}
              valueStyle={{ color: 'var(--highlight)', fontSize: 20 }}
            />
          </Col>
          <Col>
            <Statistic
              title={<span style={{ color: 'var(--muted)', fontSize: 12 }}>最新快照</span>}
              value={snapshots[0]?.version_label || '—'}
              valueStyle={{ color: 'var(--text)', fontSize: 14 }}
            />
          </Col>
          <Col>
            <div style={{ color: 'var(--muted)', fontSize: 12, lineHeight: '24px' }}>
              <WarningOutlined style={{ marginRight: 4 }} />
              快照記錄 Services / Routes / Plugins / Consumers 完整狀態，支援任意版本對比與一鍵回滾。
            </div>
          </Col>
        </Row>
      </Card>

      <Spin spinning={loading}>
        <Table
          columns={columns}
          dataSource={snapshots as any}
          rowKey="id"
          pagination={{ pageSize: 15, showSizeChanger: true }}
          locale={{ emptyText: '尚無快照，點擊「建立快照」建立第一個' }}
          size="small"
          rowSelection={{
            type: 'checkbox',
            onChange: (keys) => setSelectedKeys(keys as string[]),
          }}
        />
      </Spin>

      {/* Diff Modal */}
      <Modal
        title={<span>差異檢視 — {diffModal.title}</span>}
        open={diffModal.open}
        onCancel={() => setDiffModal({ open: false, diff: null, title: '' })}
        footer={
          <Button onClick={() => setDiffModal({ open: false, diff: null, title: '' })}>
            關閉
          </Button>
        }
        width={800}
      >
        {diffModal.diff && (
          <div>
            {renderDiffSection(' Services', diffModal.diff.services)}
            {renderDiffSection(' Routes', diffModal.diff.routes)}
            {renderDiffSection(' Plugins', diffModal.diff.plugins)}
            {renderDiffSection(' Consumers', diffModal.diff.consumers)}
            {!diffModal.diff.services?.length && !diffModal.diff.routes?.length &&
             !diffModal.diff.plugins?.length && !diffModal.diff.consumers?.length && (
              <div style={{ color: 'var(--muted)', textAlign: 'center', padding: 32 }}>
                兩個版本之間沒有差異
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  )
}



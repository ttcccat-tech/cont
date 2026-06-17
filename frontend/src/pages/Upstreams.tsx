import { useEffect, useState } from 'react'
import {
  Table, Button, Space, Tag, message, Modal, Form, Input, Popconfirm,
  Drawer, Card, Descriptions, Badge, Typography, Divider, InputNumber, Switch, Alert, Tabs
} from 'antd'
import {
  PlusOutlined, ReloadOutlined, DeleteOutlined, EditOutlined,
  NodeIndexOutlined, ArrowUpOutlined, CheckCircleOutlined, CloseCircleOutlined
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api, { KongUpstream, KongTarget, CircuitBreakerConfig } from '../api/kong'
import { useAuth } from '../context/AuthContext'

const { Text } = Typography

export default function UpstreamsPage() {
  const [upstreams, setUpstreams] = useState<KongUpstream[]>([])
  const [loading, setLoading] = useState(false)
  const [detailDrawer, setDetailDrawer] = useState(false)
  const [selectedUpstream, setSelectedUpstream] = useState<KongUpstream | null>(null)
  const [targets, setTargets] = useState<KongTarget[]>([])
  const [targetsLoading, setTargetsLoading] = useState(false)
  const [createModal, setCreateModal] = useState(false)
  const [addTargetModal, setAddTargetModal] = useState(false)
  const [editTargetModal, setEditTargetModal] = useState(false)
  const [editingTarget, setEditingTarget] = useState<KongTarget | null>(null)
  const [cbConfig, setCbConfig] = useState<CircuitBreakerConfig | null>(null)
  const [cbLoading, setCbLoading] = useState(false)
  const [cbForm] = Form.useForm()
  const [form] = Form.useForm()
  const [targetForm] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const { canWrite, canDelete } = useAuth()

  // ── Fetch upstreams ──────────────────────────────────────────────
  const fetchUpstreams = () => {
    setLoading(true)
    api.listUpstreams()
      .then(r => setUpstreams(Array.isArray(r) ? r : []))
      .catch(() => message.error('無法取得上游列表'))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchUpstreams() }, [])

  // ── Open detail drawer ──────────────────────────────────────────
  const openDetail = (upstream: KongUpstream) => {
    setSelectedUpstream(upstream)
    setDetailDrawer(true)
    fetchTargets(upstream.id!)
    fetchCircuitBreaker(upstream.id!)
  }

  const fetchTargets = (upstreamId: string) => {
    setTargetsLoading(true)
    api.listUpstreamTargets(upstreamId)
      .then(r => setTargets(Array.isArray(r) ? r : []))
      .catch(() => message.error('無法取得目標列表'))
      .finally(() => setTargetsLoading(false))
  }

  const fetchCircuitBreaker = (upstreamId: string) => {
    setCbLoading(true)
    api.getCircuitBreaker(upstreamId)
      .then(cfg => {
        setCbConfig(cfg)
        cbForm.setFieldsValue({
          enabled: cfg?.enabled ?? false,
          trip_threshold: cfg?.trip_threshold ?? 5,
          recovery_timeout: cfg?.recovery_timeout ?? 30,
          half_open_max_requests: cfg?.half_open_max_requests ?? 3,
          half_open_success_rate: cfg?.half_open_success_rate ?? 50,
        })
      })
      .catch(() => { setCbConfig(null); cbForm.resetFields() })
      .finally(() => setCbLoading(false))
  }

  const saveCircuitBreaker = () => {
    if (!selectedUpstream?.id) return
    cbForm.validateFields().then(values => {
      setSubmitting(true)
      api.setCircuitBreaker(selectedUpstream.id, {
        enabled: values.enabled,
        trip_threshold: values.trip_threshold,
        recovery_timeout: values.recovery_timeout,
        half_open_max_requests: values.half_open_max_requests,
        half_open_success_rate: values.half_open_success_rate,
      })
        .then(cfg => { setCbConfig(cfg); message.success('熔斷器設定已儲存') })
        .catch(err => message.error('儲存失敗: ' + (err.message || err)))
        .finally(() => setSubmitting(false))
    })
  }

  // ── Create upstream ──────────────────────────────────────────────
  const handleCreate = () => {
    form.validateFields().then(values => {
      setSubmitting(true)
      api.createUpstream?.(values) // backend may not have create yet, skip for now
        .then(() => {
          message.success('上游建立成功')
          setCreateModal(false)
          form.resetFields()
          fetchUpstreams()
        })
        .catch(err => message.error('建立失敗: ' + (err.message || err)))
        .finally(() => setSubmitting(false))
    })
  }

  // ── Delete upstream ──────────────────────────────────────────────
  const handleDelete = (id: string) => {
    api.deleteUpstream?.(id)
      .then(() => {
        message.success('上游已刪除')
        fetchUpstreams()
      })
      .catch(err => message.error('刪除失敗: ' + (err.message || err)))
  }

  // ── Add target ───────────────────────────────────────────────────
  const handleAddTarget = () => {
    if (!selectedUpstream?.id) return
    targetForm.validateFields().then(values => {
      setSubmitting(true)
      api.createUpstreamTarget(selectedUpstream.id, {
        target: values.target,
        weight: values.weight ?? 100,
        enabled: values.enabled ?? true,
      })
        .then(() => {
          message.success('目標已新增')
          setAddTargetModal(false)
          targetForm.resetFields()
          fetchTargets(selectedUpstream.id)
        })
        .catch(err => message.error('新增目標失敗: ' + (err.message || err)))
        .finally(() => setSubmitting(false))
    })
  }

  // ── Edit target ──────────────────────────────────────────────────
  const openEditTarget = (target: KongTarget) => {
    setEditingTarget(target)
    targetForm.setFieldsValue({
      target: target.target,
      weight: target.weight ?? 100,
      enabled: target.enabled ?? true,
    })
    setEditTargetModal(true)
  }

  const handleEditTarget = () => {
    if (!selectedUpstream?.id || !editingTarget?.id) return
    targetForm.validateFields().then(values => {
      setSubmitting(true)
      api.updateUpstreamTarget(selectedUpstream.id, editingTarget.id, {
        target: values.target,
        weight: values.weight ?? 100,
        enabled: values.enabled ?? true,
      })
        .then(() => {
          message.success('目標已更新')
          setEditTargetModal(false)
          targetForm.resetFields()
          setEditingTarget(null)
          fetchTargets(selectedUpstream.id)
        })
        .catch(err => message.error('更新目標失敗: ' + (err.message || err)))
        .finally(() => setSubmitting(false))
    })
  }

  // ── Delete target ────────────────────────────────────────────────
  const handleDeleteTarget = (targetId: string) => {
    if (!selectedUpstream?.id) return
    api.deleteUpstreamTarget(selectedUpstream.id, targetId)
      .then(() => {
        message.success('目標已刪除')
        fetchTargets(selectedUpstream.id)
      })
      .catch(err => message.error('刪除失敗: ' + (err.message || err)))
  }

  // ── Columns ──────────────────────────────────────────────────────
  const upstreamColumns: ColumnsType<KongUpstream> = [
    { title: '名稱', dataIndex: 'name', key: 'name', render: t => <Text strong>{t}</Text> },
    { title: '演算法', dataIndex: 'algorithm', key: 'algorithm', render: t => t || 'roundrobin' },
    { title: '槽位', dataIndex: 'slots', key: 'slots', render: t => t || 10000 },
    { title: '啟用', dataIndex: 'enabled', key: 'enabled', render: e => <Badge status={e ? 'success' : 'default'} text={e ? '是' : '否'} /> },
    { title: '操作', key: 'action', width: 180,
      render: (_, r) => (
        <Space>
          <Button size="small" icon={<NodeIndexOutlined />} onClick={() => openDetail(r)}>目標</Button>
          {canDelete && (
            <Popconfirm title="刪除此負載平衡後端？" onConfirm={() => handleDelete(r.id!)}>
              <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
            </Popconfirm>
          )}
        </Space>
      )
    },
  ]

  const targetColumns: ColumnsType<KongTarget> = [
    { title: '目標 (host:port)', dataIndex: 'target', key: 'target', render: t => <Text code>{t}</Text> },
    { title: '權重', dataIndex: 'weight', key: 'weight', width: 80 },
    { title: '啟用', dataIndex: 'enabled', key: 'enabled', width: 80,
      render: e => <Badge status={e ? 'success' : 'error'} text={e ? '是' : '否'} /> },
    { title: 'ID', dataIndex: 'id', key: 'id', render: id => <Text type="secondary" style={{ fontSize: 11 }}>{id?.slice(0, 8)}</Text> },
    { title: '操作', key: 'action', width: 140,
      render: (_, t) => (
        <Space>
          {canWrite && (
            <Button size="small" icon={<EditOutlined />} onClick={() => openEditTarget(t)}>編輯</Button>
          )}
          {canDelete && (
            <Popconfirm title="刪除此目標？" onConfirm={() => handleDeleteTarget(t.id!)}>
              <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
            </Popconfirm>
          )}
        </Space>
      )
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>API 負載平衡後端管理</h2>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchUpstreams}>刷新</Button>
          {canWrite && <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModal(true)}>新增後端</Button>}
        </Space>
      </div>

      {upstreams.length === 0 && !loading && (
        <Alert message="尚無負載平衡後端" description="建立 API 前端時可指定 upstream 來做負載平衡" type="info" showIcon />
      )}

      <Table columns={upstreamColumns} dataSource={upstreams} rowKey="id"
        loading={loading} pagination={{ pageSize: 10 }} />

      {/* Detail drawer */}
      <Drawer title={`負載平衡後端: ${selectedUpstream?.name || ''}`} open={detailDrawer}
        onClose={() => { setDetailDrawer(false); setTargets([]); setCbConfig(null) }} width={680}>
        <Descriptions column={2} size="small" style={{ marginBottom: 16 }}>
          <Descriptions.Item label="ID">{selectedUpstream?.id?.slice(0, 8)}</Descriptions.Item>
          <Descriptions.Item label="名稱">{selectedUpstream?.name}</Descriptions.Item>
          <Descriptions.Item label="演算法">{selectedUpstream?.algorithm || 'roundrobin'}</Descriptions.Item>
          <Descriptions.Item label="啟用">{selectedUpstream?.enabled ? '是' : '否'}</Descriptions.Item>
        </Descriptions>

        <Tabs defaultActiveKey="targets" items={[
          {
            key: 'targets',
            label: '目標 (Targets)',
            children: <>
              <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'flex-end' }}>
                {canWrite && (
                  <Button size="small" type="primary" icon={<PlusOutlined />}
                    onClick={() => setAddTargetModal(true)}>新增目標</Button>
                )}
              </div>
              <Table columns={targetColumns} dataSource={targets} rowKey="id"
                loading={targetsLoading} pagination={{ pageSize: 10 }} size="small" />
            </>
          },
          {
            key: 'circuit-breaker',
            label: '熔斷器 (Circuit Breaker)',
            children: <Form form={cbForm} layout="vertical" style={{ maxWidth: 480 }}>
              <Form.Item name="enabled" label="啟用熔斷器" valuePropName="checked">
                <Switch />
              </Form.Item>
              <Form.Item name="trip_threshold" label="熔斷閾值 (連續失敗次數)"
                extra="連續失敗達到此次數即觸發熔斷">
                <InputNumber min={1} max={100} style={{ width: '100%' }} />
              </Form.Item>
              <Form.Item name="recovery_timeout" label="恢復逾時 (秒)"
                extra="熔斷後等待多久才進入 HALF_OPEN 探測狀態">
                <InputNumber min={1} max={3600} style={{ width: '100%' }} />
              </Form.Item>
              <Form.Item name="half_open_max_requests" label="HALF_OPEN 探測次數"
                extra="HALF_OPEN 狀態下允許通過的最大請求數">
                <InputNumber min={1} max={100} style={{ width: '100%' }} />
              </Form.Item>
              <Form.Item name="half_open_success_rate" label="關閉熔斷成功率 (%)"
                extra="HALF_OPEN 探測中成功率低於此值會重回 OPEN">
                <InputNumber min={0} max={100} style={{ width: '100%' }} />
              </Form.Item>
              {canWrite && (
                <Button type="primary" onClick={saveCircuitBreaker} loading={submitting}
                  style={{ marginTop: 8 }}>
                  儲存熔斷器設定
                </Button>
              )}
            </Form>
          },
        ]} />

        {/* Add target modal */}
        <Modal title="新增目標" open={addTargetModal}
          onOk={handleAddTarget} onCancel={() => { setAddTargetModal(false); targetForm.resetFields() }}
          confirmLoading={submitting}>
          <Form form={targetForm} layout="vertical">
            <Form.Item name="target" label="目標 (host:port 或 [IPv6]:port)" rules={[{ required: true }]}>
              <Input placeholder="e.g. 192.168.1.10:8080 or [::1]:8080" />
            </Form.Item>
            <Form.Item name="weight" label="權重" initialValue={100}>
              <InputNumber min={0} max={1000} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="enabled" label="啟用" valuePropName="checked" initialValue={true}>
              <Switch />
            </Form.Item>
          </Form>
        </Modal>

        {/* Edit target modal */}
        <Modal title="編輯目標" open={editTargetModal}
          onOk={handleEditTarget} onCancel={() => { setEditTargetModal(false); targetForm.resetFields(); setEditingTarget(null) }}
          confirmLoading={submitting}>
          <Form form={targetForm} layout="vertical">
            <Form.Item name="target" label="目標" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
            <Form.Item name="weight" label="權重">
              <InputNumber min={0} max={1000} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="enabled" label="啟用" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Form>
        </Modal>
      </Drawer>

      {/* Create upstream modal (informational - backend may not have full CRUD) */}
      <Modal title="新增後端" open={createModal}
        onOk={handleCreate} onCancel={() => { setCreateModal(false); form.resetFields() }}
        confirmLoading={submitting} okText="建立">
        <Alert message="後端建立需透過 API 前端編輯介面" type="info" showIcon style={{ marginBottom: 16 }} />
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名稱" rules={[{ required: true }]}>
            <Input placeholder="my-upstream" />
          </Form.Item>
          <Form.Item name="algorithm" label="演算法" initialValue="roundrobin">
            <Input placeholder="roundrobin" />
          </Form.Item>
          <Form.Item name="slots" label="槽位" initialValue={10000}>
            <InputNumber min={10} max={65536} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

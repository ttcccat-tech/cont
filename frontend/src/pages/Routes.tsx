import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, message, Modal, Form, Input, Select, Switch, Popconfirm } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api, { KongRoute, KongService } from '../api/kong'

export default function RoutesPage() {
  const [routes, setRoutes] = useState<KongRoute[]>([])
  const [services, setServices] = useState<KongService[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<KongRoute | null>(null)
  const [form] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)

  const fetchAll = () => {
    setLoading(true)
    Promise.all([api.listRoutes(), api.listServices()])
      .then(([routesData, servicesData]) => {
        setRoutes(Array.isArray(routesData) ? routesData : (routesData?.data || []))
        setServices(Array.isArray(servicesData) ? servicesData : (servicesData?.data || []))
      })
      .catch(() => message.error('無法連接 Kong Admin API'))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchAll() }, [])

  const openCreate = () => { setEditing(null); form.resetFields(); form.setFieldsValue({ strip_path: true, preserve_host: false, plugins_enabled: false }); setModalOpen(true) }
  const openEdit = (record: KongRoute) => {
    setEditing(record)
    form.setFieldsValue({
      name: record.name,
      service_id: record.service?.id,
      protocols: record.protocols || ['http'],
      hosts: record.hosts?.join(', '),
      paths: record.paths?.join(', '),
      methods: record.methods?.join(', '),
      strip_path: record.strip_path,
      preserve_host: record.preserve_host,
    })
    setModalOpen(true)
  }

  const handleDelete = async (id: string) => {
    try {
      await api.deleteRoute(id)
      message.success('刪除成功')
      fetchAll()
    } catch { message.error('刪除失敗') }
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      const payload: Partial<KongRoute> = {
        service: values.service_id ? { id: values.service_id } : undefined,
        protocols: values.protocols,
        hosts: values.hosts ? values.hosts.split(',').map((h: string) => h.trim()) : undefined,
        paths: values.paths ? values.paths.split(',').map((p: string) => p.trim()) : undefined,
        methods: values.methods ? values.methods.split(',').map((m: string) => m.trim().toUpperCase()) : undefined,
        strip_path: values.strip_path,
        preserve_host: values.preserve_host,
      }
      if (values.name) payload.name = values.name
      if (editing) {
        await api.updateRoute(editing.id!, payload)
        message.success('更新成功')
      } else {
        await api.createRoute(payload)
        message.success('建立成功')
      }
      setModalOpen(false)
      fetchAll()
    } catch (e: any) { if (!e.errorFields) message.error('操作失敗') }
    finally { setSubmitting(false) }
  }

  const serviceName = (id: string) => services.find(s => s.id === id)?.name || id

  const columns: ColumnsType<KongRoute> = [
    { title: '名稱', dataIndex: 'name', key: 'name', render: v => v ? <Tag>{v}</Tag> : <Tag color="default">-</Tag> },
    { title: '所屬服務', key: 'service', render: (_, r) => <Tag color="cyan">{serviceName(r.service?.id || '')}</Tag> },
    { title: '協議', key: 'protocols', render: (_, r) => (r.protocols || []).map(p => <Tag key={p}>{p.toUpperCase()}</Tag>) },
    { title: '主機', dataIndex: 'hosts', key: 'hosts', render: v => Array.isArray(v) ? v.map(h => <Tag key={h}>{h}</Tag>) : '-' },
    { title: '路徑', dataIndex: 'paths', key: 'paths', render: v => Array.isArray(v) ? v.map(p => <Tag key={p}>{p}</Tag>) : '-' },
    { title: '方法', dataIndex: 'methods', key: 'methods', render: v => Array.isArray(v) ? v.map(m => <Tag key={m}>{m}</Tag>) : <Tag color="default">ANY</Tag> },
    { title: 'strip_path', dataIndex: 'strip_path', key: 'strip_path', render: v => v ? <Tag color="green">是</Tag> : <Tag color="red">否</Tag> },
    {
      title: '操作', key: 'action', width: 160,
      render: (_, record) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>編輯</Button>
          <Popconfirm title="確認刪除？" onConfirm={() => record.id && handleDelete(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
          </Popconfirm>
        </Space>
      )
    }
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>路由管理</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchAll}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增路由</Button>
        </Space>
      </div>
      <Table columns={columns} dataSource={routes as any} rowKey="id" loading={loading} pagination={{ pageSize: 10 }} locale={{ emptyText: '暫無路由' }} />

      <Modal title={editing ? '編輯路由' : '新增路由'} open={modalOpen} onOk={handleSubmit} confirmLoading={submitting} onCancel={() => setModalOpen(false)} width={580} okText={editing ? '更新' : '建立'}>
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label="路由名稱（可選）"><Input placeholder="my-route" /></Form.Item>
          <Form.Item name="service_id" label="所屬服務" rules={[{ required: true, message: '必選' }]}>
            <Select placeholder="選擇服務">{services.map(s => <Select.Option key={s.id} value={s.id}>{s.name}</Select.Option>)}</Select>
          </Form.Item>
          <Form.Item name="protocols" label="協議" initialValue={['http']}>
            <Select mode="multiple">{['http', 'https'].map(p => <Select.Option key={p} value={p}>{p.toUpperCase()}</Select.Option>)}</Select>
          </Form.Item>
          <Form.Item name="hosts" label="主機名（可選）" tooltip="多個用逗號分隔"><Input placeholder="api.example.com" /></Form.Item>
          <Form.Item name="paths" label="路徑（可選）" tooltip="多個用逗號分隔"><Input placeholder="/api" /></Form.Item>
          <Form.Item name="methods" label="HTTP 方法（可選）" tooltip="多個用逗號分隔"><Input placeholder="GET, POST" /></Form.Item>
          <Form.Item name="strip_path" label="Strip Path" valuePropName="checked" initialValue><Switch checkedChildren="開" unCheckedChildren="關" /></Form.Item>
          <Form.Item name="preserve_host" label="Preserve Host" valuePropName="checked" initialValue={false}><Switch checkedChildren="開" unCheckedChildren="關" /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

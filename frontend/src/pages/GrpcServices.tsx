import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, message, Modal, Form, Input, Switch, Popconfirm, Select, Drawer } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api, { GrpcService, GrpcMethod } from '../api/kong'
import { useAuth } from '../context/AuthContext'

export default function GrpcServices() {
  const [services, setServices] = useState<GrpcService[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<GrpcService | null>(null)
  const [form] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const { canWrite, canDelete } = useAuth()

  // Methods drawer state
  const [methodsDrawer, setMethodsDrawer] = useState(false)
  const [currentService, setCurrentService] = useState<GrpcService | null>(null)
  const [methods, setMethods] = useState<GrpcMethod[]>([])
  const [methodsLoading, setMethodsLoading] = useState(false)
  const [methodModalOpen, setMethodModalOpen] = useState(false)
  const [editingMethod, setEditingMethod] = useState<GrpcMethod | null>(null)
  const [methodForm] = Form.useForm()

  const fetchServices = () => {
    setLoading(true)
    api.listGrpcServices()
      .then(data => setServices(data))
      .catch(() => message.error('無法連接 API'))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchServices() }, [])

  const openCreate = () => { setEditing(null); form.resetFields(); setModalOpen(true) }
  const openEdit = (record: GrpcService) => {
    setEditing(record)
    form.setFieldsValue({
      name: record.name,
      package: record.package || '',
      proto_file: record.proto_file || '',
      upstream_id: record.upstream_id || '',
      enabled: record.enabled ?? true,
    })
    setModalOpen(true)
  }

  const handleDelete = async (id: string) => {
    try {
      await api.deleteGrpcService(id)
      message.success('刪除成功')
      fetchServices()
    } catch (err: any) {
      message.error('刪除失敗: ' + (err.message || ''))
    }
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      if (editing?.id) {
        await api.updateGrpcService(editing.id, values)
        message.success('更新成功')
      } else {
        await api.createGrpcService(values)
        message.success('建立成功')
      }
      setModalOpen(false)
      fetchServices()
    } catch (e: any) {
      if (e.errorFields) return
      message.error('操作失敗: ' + (e.message || '未知錯誤'))
    } finally {
      setSubmitting(false)
    }
  }

  // Methods drawer
  const openMethodsDrawer = (svc: GrpcService) => {
    setCurrentService(svc)
    setMethodsDrawer(true)
    fetchMethods(svc.id!)
  }

  const fetchMethods = (serviceId: string) => {
    setMethodsLoading(true)
    api.listGrpcMethods(serviceId)
      .then(data => setMethods(data))
      .catch(() => message.error('無法載入方法'))
      .finally(() => setMethodsLoading(false))
  }

  const openCreateMethod = () => { setEditingMethod(null); methodForm.resetFields(); setMethodModalOpen(true) }

  const handleDeleteMethod = async (methodId: string) => {
    if (!currentService?.id) return
    try {
      await api.deleteGrpcMethod(currentService.id, methodId)
      message.success('刪除成功')
      fetchMethods(currentService.id)
    } catch (err: any) {
      message.error('刪除失敗: ' + (err.message || ''))
    }
  }

  const handleMethodSubmit = async () => {
    if (!currentService?.id) return
    try {
      const values = await methodForm.validateFields()
      setSubmitting(true)
      if (editingMethod?.id) {
        // Update not implemented — delete + create for simplicity
        await api.deleteGrpcMethod(currentService.id, editingMethod.id)
      }
      await api.createGrpcMethod(currentService.id, values)
      message.success('建立成功')
      setMethodModalOpen(false)
      fetchMethods(currentService.id)
    } catch (e: any) {
      if (e.errorFields) return
      message.error('操作失敗: ' + (e.message || '未知錯誤'))
    } finally {
      setSubmitting(false)
    }
  }

  const columns: ColumnsType<GrpcService> = [
    { title: '名稱', dataIndex: 'name', key: 'name', render: v => <Tag color="purple">{v}</Tag> },
    { title: 'Package', dataIndex: 'package', key: 'package', render: v => <code style={{ fontSize: 12 }}>{v || '-'}</code> },
    { title: 'Upstream', dataIndex: 'upstream_id', key: 'upstream_id', render: v => v ? <code style={{ fontSize: 11 }}>{v.slice(0, 8)}...</code> : <Tag>-</Tag> },
    {
      title: 'Proto',
      dataIndex: 'proto_file',
      key: 'proto_file',
      ellipsis: true,
      render: v => v ? <code style={{ fontSize: 10 }}>{v.slice(0, 40)}...</code> : <Tag color="default">-</Tag>
    },
    {
      title: '操作',
      key: 'action',
      width: 260,
      render: (_, record) => (
        <Space>
          <Button size="small" onClick={() => openMethodsDrawer(record)}>方法</Button>
          {canWrite('services') && (
            <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>編輯</Button>
          )}
          {canDelete('services') && (
            <Popconfirm title="確認刪除？" onConfirm={() => record.id && handleDelete(record.id)}>
              <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
            </Popconfirm>
          )}
        </Space>
      )
    }
  ]

  const methodColumns: ColumnsType<GrpcMethod> = [
    { title: '方法名稱', dataIndex: 'name', key: 'name', render: v => <Tag color="blue">{v}</Tag> },
    { title: 'Type', dataIndex: 'method_type', key: 'method_type', render: v => <Tag>{v || 'unary'}</Tag> },
    { title: 'Input', dataIndex: 'input_type', key: 'input_type', render: v => <code style={{ fontSize: 11 }}>{v || '-'}</code> },
    { title: 'Output', dataIndex: 'output_type', key: 'output_type', render: v => <code style={{ fontSize: 11 }}>{v || '-'}</code> },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_, record) => (
        <Space>
          {canDelete('services') && (
            <Popconfirm title="確認刪除？" onConfirm={() => record.id && handleDeleteMethod(record.id)}>
              <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
            </Popconfirm>
          )}
        </Space>
      )
    }
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>gRPC 服務管理</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchServices}>刷新</Button>
          {canWrite('services') && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增 gRPC 服務</Button>
          )}
        </Space>
      </div>

      <Table
        columns={columns}
        dataSource={services as any}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 10 }}
        locale={{ emptyText: '暫無 gRPC 服務，點擊「新增 gRPC 服務」建立第一個' }}
      />

      <Modal
        title={editing ? '編輯 gRPC 服務' : '新增 gRPC 服務'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        width={560}
        footer={null}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }} onFinish={handleSubmit}>
          <Form.Item name="name" label="服務名稱" rules={[{ required: true, message: '必填' }]}>
            <Input placeholder="helloworld" />
          </Form.Item>
          <Form.Item name="package" label="Proto Package" tooltip="例: helloworld, my.company.api">
            <Input placeholder="helloworld" />
          </Form.Item>
          <Form.Item name="upstream_id" label="Upstream ID" tooltip="可選，關聯到現有的 Upstream">
            <Input placeholder="可留空" />
          </Form.Item>
          <Form.Item name="proto_file" label="Proto 文件內容" tooltip="可選，paste .proto 檔案內容">
            <Input.TextArea rows={6} placeholder="syntax = &quot;proto3&quot;; ..." />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, marginTop: 8 }}>
            <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
              <Button onClick={() => setModalOpen(false)}>取消</Button>
              <Button type="primary" htmlType="submit" loading={submitting}>
                {editing ? '更新' : '建立'}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title={`gRPC 方法 — ${currentService?.name}`}
        open={methodsDrawer}
        onClose={() => { setMethodsDrawer(false); setCurrentService(null) }}
        width={720}
        extra={
          canWrite('services') ? (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreateMethod}>新增方法</Button>
          ) : null
        }
      >
        <Table
          columns={methodColumns}
          dataSource={methods as any}
          rowKey="id"
          loading={methodsLoading}
          pagination={{ pageSize: 10 }}
          locale={{ emptyText: '暫無方法，點擊「新增方法」建立' }}
        />
      </Drawer>

      <Modal
        title="新增 gRPC 方法"
        open={methodModalOpen}
        onCancel={() => setMethodModalOpen(false)}
        width={480}
        footer={null}
      >
        <Form form={methodForm} layout="vertical" style={{ marginTop: 16 }} onFinish={handleMethodSubmit}>
          <Form.Item name="name" label="方法名稱" rules={[{ required: true, message: '必填' }]}>
            <Input placeholder="SayHello" />
          </Form.Item>
          <Form.Item name="method_type" label="方法類型" initialValue="unary">
            <Select placeholder="選擇方法類型">
              <Select.Option value="unary">Unary (一般 RPC)</Select.Option>
              <Select.Option value="client_streaming">Client Streaming</Select.Option>
              <Select.Option value="server_streaming">Server Streaming</Select.Option>
              <Select.Option value="bidirectional">Bidirectional Streaming</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="input_type" label="Input Type" tooltip="例: HelloRequest">
            <Input placeholder="HelloRequest" />
          </Form.Item>
          <Form.Item name="output_type" label="Output Type" tooltip="例: HelloReply">
            <Input placeholder="HelloReply" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
              <Button onClick={() => setMethodModalOpen(false)}>取消</Button>
              <Button type="primary" htmlType="submit" loading={submitting}>建立</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

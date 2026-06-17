import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, message, Modal, Form, Input, InputNumber, Popconfirm, Select } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api, { KongService, KongUpstream } from '../api/kong'
import { useAuth } from '../context/AuthContext'

export default function Services() {
  const [services, setServices] = useState<KongService[]>([])
  const [upstreams, setUpstreams] = useState<KongUpstream[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<KongService | null>(null)
  const [form] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const { canWrite, canDelete } = useAuth()

  const fetchServices = () => {
    setLoading(true)
    api.listServices()
      .then(data => setServices(data))
      .catch(() => message.error('無法連接 Cont Admin API'))
      .finally(() => setLoading(false))
  }

  const fetchUpstreams = () => {
    api.listUpstreams()
      .then(setUpstreams)
      .catch(() => message.error('無法載入後端列表'))
  }

  useEffect(() => {
    fetchServices()
  }, [])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    fetchUpstreams()
    setModalOpen(true)
  }

  const openEdit = (record: KongService) => {
    setEditing(record)
    fetchUpstreams()
    form.setFieldsValue({
      name: record.name,
      protocol: record.protocol || 'http',
      upstream_id: record.upstream_id || undefined,
      retries: record.retries ?? 5,
      connect_timeout: record.connect_timeout ?? 60000,
      read_timeout: record.read_timeout ?? 60000,
      write_timeout: record.write_timeout ?? 60000,
    })
    setModalOpen(true)
  }

  const handleDelete = async (id: string) => {
    try {
      await api.deleteService(id)
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
      const payload: any = {
        name: values.name,
        protocol: values.protocol || 'http',
        retries: values.retries ?? 5,
        connect_timeout: values.connect_timeout ?? 60000,
        read_timeout: values.read_timeout ?? 60000,
        write_timeout: values.write_timeout ?? 60000,
      }
      if (values.upstream_id) {
        payload.upstream_id = values.upstream_id
      }
      if (values.health_url) payload.health_url = values.health_url
      if (values.doc_url) payload.doc_url = values.doc_url
      if (editing?.id) {
        await api.updateService(editing.id, payload)
        message.success('更新成功')
      } else {
        await api.createService(payload)
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

  const upstreamName = (id?: string) => {
    if (!id) return '—'
    const u = upstreams.find(u => u.id === id)
    return u ? u.name : id
  }

  const columns: ColumnsType<KongService> = [
    { title: '名稱', dataIndex: 'name', key: 'name', render: v => <Tag color="blue">{v}</Tag> },
    {
      title: 'API 目標(後端)',
      key: 'upstream',
      render: (_, r) => r.upstream_id ? <Tag color="green">{upstreamName(r.upstream_id)}</Tag> : <Tag color="default">—</Tag>
    },
    { title: '重試次數', dataIndex: 'retries', key: 'retries' },
    {
      title: '超時 (ms)',
      key: 'timeout',
      render: (_, r) => `${r.connect_timeout}/${r.read_timeout}/${r.write_timeout}`
    },
    {
      title: '操作',
      key: 'action',
      width: 160,
      render: (_, record) => (
        <Space>
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

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>API 前端管理</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchServices}>刷新</Button>
          {canWrite('services') && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增 API 前端</Button>
          )}
        </Space>
      </div>

      <Table
        columns={columns}
        dataSource={services as any}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 10 }}
        locale={{ emptyText: '暫無 API 前端，點擊「新增 API 前端」建立第一個' }}
      />

      <Modal
        title={editing ? '編輯 API 前端' : '新增 API 前端'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        width={560}
        footer={null}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }} onFinish={handleSubmit}>
          <Form.Item name="name" label="服務名稱" rules={[{ required: true, message: '必填' }]}>
            <Input placeholder="my-api-service" />
          </Form.Item>
          <Form.Item name="upstream_id" label="API 目標(後端)">
            <Select
              placeholder="請選擇後端（可選）"
              allowClear
              showSearch
              optionFilterProp="label"
              options={upstreams.map(u => ({ value: u.id, label: u.name }))}
            />
          </Form.Item>
          <Form.Item name="protocol" label="協議" initialValue="http">
            <Select options={[{ value: 'http', label: 'HTTP' }, { value: 'https', label: 'HTTPS' }]} />
          </Form.Item>
          <Form.Item name="retries" label="重試次數" initialValue={5}>
            <InputNumber min={0} max={100} style={{ width: 120 }} />
          </Form.Item>
          <Form.Item name="connect_timeout" label="連接超時 (ms)" initialValue={60000}>
            <InputNumber min={100} max={600000} style={{ width: 140 }} />
          </Form.Item>
          <Form.Item name="read_timeout" label="讀取超時 (ms)" initialValue={60000}>
            <InputNumber min={100} max={600000} style={{ width: 140 }} />
          </Form.Item>
          <Form.Item name="write_timeout" label="寫入超時 (ms)" initialValue={60000}>
            <InputNumber min={100} max={600000} style={{ width: 140 }} />
          </Form.Item>
          <Form.Item name="health_url" label="健康檢查 URL" tooltip="例: https://api.example.com/health">
            <Input placeholder="https://api.example.com/health" />
          </Form.Item>
          <Form.Item name="doc_url" label="API 文件 URL">
            <Input placeholder="https://api.example.com/docs" />
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
    </div>
  )
}
